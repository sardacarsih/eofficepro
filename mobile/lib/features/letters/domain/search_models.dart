class LetterSearchResult {
  const LetterSearchResult({
    required this.id,
    required this.companyCode,
    required this.letterTypeCode,
    required this.subject,
    required this.status,
    required this.classification,
    required this.creatorName,
    required this.origin,
    required this.snippet,
    required this.updatedAt,
    this.letterNumber,
    this.publishedAt,
  });

  final String id;
  final String companyCode;
  final String letterTypeCode;
  final String? letterNumber;
  final String subject;
  final String status;
  final String classification;
  final String creatorName;
  final String origin;
  final String snippet;
  final String? publishedAt;
  final String updatedAt;

  factory LetterSearchResult.fromJson(Map<String, dynamic> json) {
    return LetterSearchResult(
      id: json['id'] as String? ?? '',
      companyCode: json['company_code'] as String? ?? '',
      letterTypeCode: json['letter_type_code'] as String? ?? '',
      letterNumber: json['letter_number'] as String?,
      subject: json['subject'] as String? ?? '',
      status: json['status'] as String? ?? '',
      classification: json['classification'] as String? ?? '',
      creatorName: json['creator_name'] as String? ?? '',
      origin: json['origin'] as String? ?? '',
      snippet: json['snippet'] as String? ?? '',
      publishedAt: json['published_at'] as String?,
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }
}
