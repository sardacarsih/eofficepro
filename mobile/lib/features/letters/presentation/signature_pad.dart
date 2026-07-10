import 'dart:convert';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

class SignaturePad extends StatefulWidget {
  const SignaturePad({
    required this.controller,
    super.key,
  });

  final SignaturePadController controller;

  @override
  State<SignaturePad> createState() => _SignaturePadState();
}

class _SignaturePadState extends State<SignaturePad> {
  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_handleChanged);
  }

  @override
  void didUpdateWidget(SignaturePad oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller) {
      oldWidget.controller.removeListener(_handleChanged);
      widget.controller.addListener(_handleChanged);
    }
  }

  @override
  void dispose() {
    widget.controller.removeListener(_handleChanged);
    super.dispose();
  }

  void _handleChanged() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Semantics(
      label: 'Area tanda tangan',
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onPanStart: (details) =>
            widget.controller.beginStroke(details.localPosition),
        onPanUpdate: (details) =>
            widget.controller.appendPoint(details.localPosition),
        onPanEnd: (_) => widget.controller.endStroke(),
        child: CustomPaint(
          painter: _SignaturePainter(
            strokes: widget.controller.strokes,
            strokeColor: Colors.black,
            guideColor: colorScheme.outlineVariant,
          ),
          child: const SizedBox.expand(),
        ),
      ),
    );
  }
}

class SignaturePadController extends ChangeNotifier {
  final List<List<Offset>> _strokes = <List<Offset>>[];

  List<List<Offset>> get strokes =>
      _strokes.map((stroke) => List<Offset>.unmodifiable(stroke)).toList();

  bool get hasSignature => _strokes.any((stroke) => stroke.length > 1);

  void beginStroke(Offset point) {
    _strokes.add(<Offset>[point]);
    notifyListeners();
  }

  void appendPoint(Offset point) {
    if (_strokes.isEmpty) {
      beginStroke(point);
      return;
    }
    _strokes.last.add(point);
    notifyListeners();
  }

  void endStroke() {
    if (_strokes.isNotEmpty && _strokes.last.length == 1) {
      final point = _strokes.last.single;
      _strokes.last.add(point.translate(0.1, 0.1));
      notifyListeners();
    }
  }

  void clear() {
    if (_strokes.isEmpty) return;
    _strokes.clear();
    notifyListeners();
  }
}

class SignatureCaptureDialog extends StatefulWidget {
  const SignatureCaptureDialog({super.key});

  @override
  State<SignatureCaptureDialog> createState() => _SignatureCaptureDialogState();
}

class _SignatureCaptureDialogState extends State<SignatureCaptureDialog> {
  final _boundaryKey = GlobalKey();
  late final SignaturePadController _controller;
  var _exporting = false;

  @override
  void initState() {
    super.initState();
    _controller = SignaturePadController()..addListener(_handleChanged);
  }

  @override
  void dispose() {
    _controller
      ..removeListener(_handleChanged)
      ..dispose();
    super.dispose();
  }

  void _handleChanged() => setState(() {});

  Future<void> _submit() async {
    if (!_controller.hasSignature || _exporting) return;
    setState(() => _exporting = true);
    try {
      final boundary = _boundaryKey.currentContext?.findRenderObject()
          as RenderRepaintBoundary?;
      if (boundary == null) return;
      final image = await boundary.toImage(pixelRatio: 2);
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (!mounted || data == null) return;
      Navigator.of(context).pop(base64Encode(data.buffer.asUint8List()));
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final size = MediaQuery.sizeOf(context);
    final width = (size.width - 48).clamp(320.0, 760.0).toDouble();
    final height = (size.height * 0.46).clamp(260.0, 420.0).toDouble();

    return Dialog(
      insetPadding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: width),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Tanda tangan',
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 12),
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: Colors.white,
                    border: Border.all(color: colorScheme.outlineVariant),
                  ),
                  child: SizedBox(
                    height: height,
                    child: RepaintBoundary(
                      key: _boundaryKey,
                      child: ColoredBox(
                        color: Colors.white,
                        child: SignaturePad(controller: _controller),
                      ),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              Wrap(
                alignment: WrapAlignment.end,
                spacing: 8,
                runSpacing: 8,
                children: [
                  TextButton(
                    onPressed: _exporting || !_controller.hasSignature
                        ? null
                        : _controller.clear,
                    child: const Text('Bersihkan'),
                  ),
                  TextButton(
                    onPressed:
                        _exporting ? null : () => Navigator.of(context).pop(),
                    child: const Text('Batal'),
                  ),
                  FilledButton.icon(
                    onPressed: _controller.hasSignature && !_exporting
                        ? _submit
                        : null,
                    icon: _exporting
                        ? const SizedBox.square(
                            dimension: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.draw),
                    label: const Text('Tanda tangani'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SignaturePainter extends CustomPainter {
  const _SignaturePainter({
    required this.strokes,
    required this.strokeColor,
    required this.guideColor,
  });

  final List<List<Offset>> strokes;
  final Color strokeColor;
  final Color guideColor;

  @override
  void paint(Canvas canvas, Size size) {
    final guidePaint = Paint()
      ..color = guideColor
      ..strokeWidth = 1;
    canvas.drawLine(
      Offset(24, size.height * 0.72),
      Offset(size.width - 24, size.height * 0.72),
      guidePaint,
    );

    final strokePaint = Paint()
      ..color = strokeColor
      ..strokeWidth = 3
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round
      ..style = PaintingStyle.stroke;

    for (final stroke in strokes) {
      if (stroke.length < 2) continue;
      final path = Path()..moveTo(stroke.first.dx, stroke.first.dy);
      for (final point in stroke.skip(1)) {
        path.lineTo(point.dx, point.dy);
      }
      canvas.drawPath(path, strokePaint);
    }
  }

  @override
  bool shouldRepaint(covariant _SignaturePainter oldDelegate) {
    return oldDelegate.strokes != strokes ||
        oldDelegate.strokeColor != strokeColor ||
        oldDelegate.guideColor != guideColor;
  }
}
