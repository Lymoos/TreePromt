// Веб-воркер Drift: SQLite (WASM) работает в нём, а не в потоке интерфейса;
// несколько вкладок делят одну базу. Сборка: см. README.
import 'package:drift/wasm.dart';

void main() => WasmDatabase.workerMainForOpen();
