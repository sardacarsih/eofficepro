import 'dart:convert';

import 'package:html/dom.dart';
import 'package:html/parser.dart' as html_parser;

const _htmlEscape = HtmlEscape(HtmlEscapeMode.element);

final _recipientPlaceholderPattern = RegExp(
  r'\{\{\s*tujuan\s*\}\}',
  caseSensitive: false,
);

final _templateRecipientParagraphPattern = RegExp(
  r'^yth\.?\s*\{\{\s*tujuan\s*\}\}$',
  caseSensitive: false,
);

const _blockElements = <String>{
  'address',
  'article',
  'aside',
  'blockquote',
  'div',
  'footer',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'header',
  'li',
  'main',
  'nav',
  'ol',
  'p',
  'pre',
  'section',
  'table',
  'tr',
  'ul',
};

const _ignoredElements = <String>{'script', 'style', 'template'};
const _richEditableElements = <String>{
  'p',
  'br',
  'div',
  'strong',
  'b',
  'em',
  'i',
  'u',
  'ul',
  'ol',
  'li',
};

/// Returns true when the stored HTML uses markup beyond the natively
/// editable subset, so editing it would discard formatting.
bool hasUnsupportedLetterFormatting(String source) {
  if (source.trim().isEmpty) return false;
  final fragment = html_parser.parseFragment(source);
  for (final element in fragment.querySelectorAll('*')) {
    if (!_richEditableElements.contains(element.localName) ||
        element.attributes.isNotEmpty) {
      return true;
    }
    if ((element.localName == 'ul' || element.localName == 'ol') &&
        _hasListAncestor(element)) {
      return true;
    }
  }
  return false;
}

bool _hasListAncestor(Element element) {
  var parent = element.parent;
  while (parent != null) {
    final name = parent.localName;
    if (name == 'ul' || name == 'ol' || name == 'li') return true;
    parent = parent.parent;
  }
  return false;
}

/// Converts stored letter HTML into editable plain text.
String letterHtmlToPlainText(String source) {
  if (source.trim().isEmpty) return '';

  final fragment = html_parser.parseFragment(source);
  for (final paragraph in fragment.querySelectorAll('p').toList()) {
    final normalizedText =
        paragraph.text.replaceAll(RegExp(r'\s+'), ' ').trim();
    if (_templateRecipientParagraphPattern.hasMatch(normalizedText)) {
      paragraph.remove();
    }
  }

  final buffer = StringBuffer();
  for (final node in fragment.nodes) {
    _appendPlainText(node, buffer);
  }

  return buffer
      .toString()
      .replaceAll('\u00a0', ' ')
      .replaceAll(_recipientPlaceholderPattern, '')
      .replaceAll(RegExp(r'[ \t]+'), ' ')
      .replaceAll(RegExp(r' *\n *'), '\n')
      .replaceAll(RegExp(r'\n{3,}'), '\n\n')
      .trim();
}

/// Converts editable plain text into HTML safe for the letter API.
String plainTextToLetterHtml(String source) {
  final normalized =
      source.replaceAll('\r\n', '\n').replaceAll('\r', '\n').trim();
  if (normalized.isEmpty) return '';

  return normalized.split('\n').map((line) {
    if (line.isEmpty) return '<p><br></p>';
    return '<p>${_htmlEscape.convert(line)}</p>';
  }).join();
}

/// Converts editable letter HTML into the canonical form used to seed the
/// native editor and to detect edits by string equality.
///
/// The canonical dialect is attribute-free `p`, `strong`, `em`, `u`, `ul`,
/// `ol`, `li` blocks with `<p><br></p>` for blank lines. The native Kotlin
/// codec (LetterHtmlCodec.kt) must serialize back to this exact form; its
/// golden tests mirror the cases in letter_body_codec_test.dart.
String normalizeEditableLetterHtml(String source) {
  if (source.trim().isEmpty) return '';

  final fragment = html_parser.parseFragment(source);
  for (final paragraph in fragment.querySelectorAll('p').toList()) {
    final normalizedText =
        paragraph.text.replaceAll(RegExp(r'\s+'), ' ').trim();
    if (_templateRecipientParagraphPattern.hasMatch(normalizedText)) {
      paragraph.remove();
    }
  }

  final builder = _CanonicalBuilder();
  for (final node in fragment.nodes) {
    builder.addNode(node, const _InlineStyle());
  }
  return builder.serialize();
}

class _InlineStyle {
  const _InlineStyle({
    this.bold = false,
    this.italic = false,
    this.underline = false,
  });

  final bool bold;
  final bool italic;
  final bool underline;

  bool sameAs(_InlineStyle other) =>
      bold == other.bold &&
      italic == other.italic &&
      underline == other.underline;
}

class _Run {
  _Run(this.text, this.style);

  String text;
  final _InlineStyle style;
}

sealed class _CanonicalBlock {}

class _ParagraphBlock extends _CanonicalBlock {
  _ParagraphBlock(this.runs);

  final List<_Run> runs;
}

class _ListBlock extends _CanonicalBlock {
  _ListBlock(this.ordered) : items = [];

  final bool ordered;
  final List<List<_Run>> items;
}

class _CanonicalBuilder {
  _CanonicalBuilder({_LineCollector? paragraphs})
      : _paragraphs = paragraphs ?? _LineCollector();

  final List<_CanonicalBlock> _blocks = [];
  final _LineCollector _paragraphs;

  void addNode(Node node, _InlineStyle style) {
    if (node is Text) {
      _paragraphs.addText(node.data, style);
      return;
    }
    if (node is! Element || _ignoredElements.contains(node.localName)) return;

    switch (node.localName) {
      case 'br':
        _paragraphs.addLineBreak();
      case 'ul' || 'ol':
        _flushParagraphs();
        _addList(node, node.localName == 'ol', style);
      case 'strong' || 'b':
        _addChildren(node, _styledWith(style, bold: true));
      case 'em' || 'i':
        _addChildren(node, _styledWith(style, italic: true));
      case 'u':
        _addChildren(node, _styledWith(style, underline: true));
      case 'p' || 'div':
        _paragraphs.openBlock();
        _addChildren(node, style);
        _paragraphs.closeBlock();
      default:
        if (_blockElements.contains(node.localName)) {
          _paragraphs.openBlock();
          _addChildren(node, style);
          _paragraphs.closeBlock();
        } else {
          _addChildren(node, style);
        }
    }
  }

  String serialize() {
    _flushParagraphs();
    final merged = <_CanonicalBlock>[];
    for (final block in _blocks) {
      final last = merged.isEmpty ? null : merged.last;
      if (block is _ListBlock && last is _ListBlock &&
          last.ordered == block.ordered) {
        last.items.addAll(block.items);
      } else {
        merged.add(block);
      }
    }
    final buffer = StringBuffer();
    for (final block in merged) {
      switch (block) {
        case _ParagraphBlock(:final runs):
          buffer.write(
            runs.isEmpty ? '<p><br></p>' : '<p>${_serializeRuns(runs)}</p>',
          );
        case _ListBlock(:final ordered, :final items):
          if (items.isEmpty) continue;
          final tag = ordered ? 'ol' : 'ul';
          buffer.write('<$tag>');
          for (final item in items) {
            buffer.write(
              item.isEmpty ? '<li><br></li>' : '<li>${_serializeRuns(item)}</li>',
            );
          }
          buffer.write('</$tag>');
      }
    }
    return buffer.toString();
  }

  void _addChildren(Element element, _InlineStyle style) {
    for (final child in element.nodes) {
      addNode(child, style);
    }
  }

  void _addList(Element list, bool ordered, _InlineStyle style) {
    final block = _ListBlock(ordered);
    for (final child in list.children) {
      if (child.localName != 'li') continue;
      final collector = _LineCollector();
      final itemBuilder = _CanonicalBuilder(paragraphs: collector);
      collector.openBlock();
      for (final node in child.nodes) {
        itemBuilder.addNode(node, style);
      }
      collector.closeBlock();
      block.items.addAll(collector.lines);
      // Nested lists are outside the editable subset; if one slips through,
      // flatten its items into the parent list.
      for (final nested in itemBuilder._blocks.whereType<_ListBlock>()) {
        block.items.addAll(nested.items);
      }
    }
    if (block.items.isNotEmpty) _blocks.add(block);
  }

  void _flushParagraphs() {
    _paragraphs.closeBlock();
    for (final line in _paragraphs.takeLines()) {
      _blocks.add(_ParagraphBlock(line));
    }
  }
}

class _LineCollector {
  final List<List<_Run>> lines = [];
  List<_Run> _runs = [];
  bool _active = false;
  bool _brInBlock = false;

  void openBlock() {
    closeBlock();
    _active = true;
  }

  void closeBlock() {
    if (!_active) return;
    final line = _finalizeLine(_runs);
    if (line.isNotEmpty || !_brInBlock) lines.add(line);
    _runs = [];
    _active = false;
    _brInBlock = false;
  }

  void addLineBreak() {
    _active = true;
    _brInBlock = true;
    lines.add(_finalizeLine(_runs));
    _runs = [];
  }

  void addText(String data, _InlineStyle style) {
    final text = data
        .replaceAll('\u00a0', ' ')
        .replaceAll(_recipientPlaceholderPattern, '')
        .replaceAll(RegExp(r'\s+'), ' ');
    if (text.isEmpty) return;
    if (!_active) {
      if (text.trim().isEmpty) return;
      _active = true;
    }
    _runs.add(_Run(text, style));
  }

  List<List<_Run>> takeLines() {
    final result = List<List<_Run>>.from(lines);
    lines.clear();
    return result;
  }
}

List<_Run> _finalizeLine(List<_Run> runs) {
  final collapsed = <_Run>[];
  for (final run in runs) {
    var text = run.text;
    if (collapsed.isEmpty) {
      text = text.trimLeft();
    } else if (collapsed.last.text.endsWith(' ') && text.startsWith(' ')) {
      text = text.replaceFirst(RegExp(r'^ +'), '');
    }
    if (text.isEmpty) continue;
    final last = collapsed.isEmpty ? null : collapsed.last;
    if (last != null && last.style.sameAs(run.style)) {
      last.text += text;
    } else {
      collapsed.add(_Run(text, run.style));
    }
  }
  while (collapsed.isNotEmpty) {
    final last = collapsed.last;
    last.text = last.text.trimRight();
    if (last.text.isEmpty) {
      collapsed.removeLast();
    } else {
      break;
    }
  }
  return collapsed;
}

_InlineStyle _styledWith(
  _InlineStyle style, {
  bool bold = false,
  bool italic = false,
  bool underline = false,
}) {
  return _InlineStyle(
    bold: style.bold || bold,
    italic: style.italic || italic,
    underline: style.underline || underline,
  );
}

String _serializeRuns(List<_Run> runs) => _serializeLevel(runs, 0);

const _inlineTags = ['strong', 'em', 'u'];

String _serializeLevel(List<_Run> runs, int level) {
  if (level == _inlineTags.length) {
    return runs.map((run) => _htmlEscape.convert(run.text)).join();
  }
  bool flagOf(_Run run) => switch (level) {
        0 => run.style.bold,
        1 => run.style.italic,
        _ => run.style.underline,
      };
  final buffer = StringBuffer();
  var index = 0;
  while (index < runs.length) {
    final flag = flagOf(runs[index]);
    var end = index;
    while (end < runs.length && flagOf(runs[end]) == flag) {
      end++;
    }
    final inner = _serializeLevel(runs.sublist(index, end), level + 1);
    buffer.write(
      flag ? '<${_inlineTags[level]}>$inner</${_inlineTags[level]}>' : inner,
    );
    index = end;
  }
  return buffer.toString();
}

void _appendPlainText(Node node, StringBuffer buffer) {
  if (node is Text) {
    if (node.data.trim().isEmpty && node.data.contains(RegExp(r'[\r\n]'))) {
      return;
    }
    buffer.write(node.data);
    return;
  }
  if (node is! Element || _ignoredElements.contains(node.localName)) return;

  if (node.localName == 'br') {
    buffer.write('\n');
    return;
  }

  for (final child in node.nodes) {
    _appendPlainText(child, buffer);
  }

  if (node.localName == 'td' || node.localName == 'th') {
    buffer.write('\t');
  } else if (_blockElements.contains(node.localName)) {
    buffer.write('\n');
  }
}
