import 'package:eoffice_mobile/features/letters/domain/draft_models.dart';

extension LetterClassificationLabel on LetterClassification {
  String get displayLabel => switch (this) {
        LetterClassification.biasa => 'Biasa',
        LetterClassification.terbatas => 'Terbatas',
        LetterClassification.rahasia => 'Rahasia',
      };
}
