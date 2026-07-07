import 'package:flutter/material.dart';

void main() => runApp(const EOfficeApp());

class EOfficeApp extends StatelessWidget {
  const EOfficeApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'eOffice Pro',
      theme: ThemeData(
        colorSchemeSeed: const Color(0xFF2E7D4F), // hijau KSK
        useMaterial3: true,
      ),
      home: const Scaffold(
        body: Center(
          child: Text('eOffice Pro — scaffold awal.\nLayar login menyusul di Epic E09.'),
        ),
      ),
    );
  }
}
