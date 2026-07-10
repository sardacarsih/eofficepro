@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "AVD_NAME=Pixel_Tablet_API_35"
set "PROJECT_DIR=%~dp0mobile"
set "WAIT_SECONDS=180"
set "API_BASE_URL=http://10.0.2.2:8080/api/v1"
set "API_HOST=10.0.2.2"
set "API_PORT=8080"

if not exist "%PROJECT_DIR%\pubspec.yaml" (
  echo Flutter project not found: "%PROJECT_DIR%"
  exit /b 1
)

where flutter >nul 2>nul
if errorlevel 1 (
  echo Flutter command not found. Make sure Flutter is installed and available in PATH.
  exit /b 1
)

where adb >nul 2>nul
if errorlevel 1 (
  echo adb command not found. Make sure Android platform-tools is available in PATH.
  exit /b 1
)

call :find_tablet
if defined DEVICE_ID (
  call :run_app %*
  exit /b !ERRORLEVEL!
)

echo Launching Android tablet emulator: %AVD_NAME%
call flutter emulators --launch %AVD_NAME%
if errorlevel 1 (
  echo Emulator launch returned an error. Checking whether "%AVD_NAME%" is already running...
)

echo Waiting for tablet emulator to be ready...
set /a "TRIES=WAIT_SECONDS / 5"

:find_device
call :find_tablet
if defined DEVICE_ID (
  call :run_app %*
  exit /b !ERRORLEVEL!
)

if %TRIES% leq 0 (
  echo Timed out waiting for emulator "%AVD_NAME%".
  exit /b 1
)

set /a "TRIES-=1"
timeout /t 5 /nobreak >nul
goto :find_device

:run_app
echo Tablet emulator is ready: %DEVICE_ID%
call :prepare_emulator_network
if errorlevel 1 exit /b 1

call :check_api
if errorlevel 1 exit /b 1

call :check_fcm_prerequisites
if errorlevel 1 exit /b 1

if /i "%~1"=="--check" (
  echo Preflight checks passed. Emulator is ready for API development and FCM client testing.
  exit /b 0
)

pushd "%PROJECT_DIR%"
call flutter run -d %DEVICE_ID% --dart-define=API_BASE_URL=%API_BASE_URL% --dart-define=FIREBASE_MESSAGING_ENABLED=true
set "RUN_EXIT=%ERRORLEVEL%"
popd

exit /b %RUN_EXIT%

:prepare_emulator_network
echo Preparing emulator network...
adb -s %DEVICE_ID% shell settings put global airplane_mode_on 0 >nul 2>nul
adb -s %DEVICE_ID% shell svc wifi enable >nul 2>nul
adb -s %DEVICE_ID% shell svc data enable >nul 2>nul
adb -s %DEVICE_ID% shell settings delete global http_proxy >nul 2>nul
adb -s %DEVICE_ID% shell settings put global private_dns_mode off >nul 2>nul

adb -s %DEVICE_ID% shell dumpsys connectivity | findstr /c:"VALIDATED" >nul 2>nul
if errorlevel 1 (
  echo Emulator network is not validated. Check host internet connection or restart the AVD.
  exit /b 1
)

echo Emulator internet is available.
exit /b 0

:check_api
echo Checking development API from emulator: %API_BASE_URL%
adb -s %DEVICE_ID% shell toybox nc -z -w 3 %API_HOST% %API_PORT% >nul 2>nul
if errorlevel 1 (
  echo Development API is not reachable from emulator.
  echo Expected API: %API_BASE_URL%
  echo Start the backend on Windows host port %API_PORT%, then run this script again.
  exit /b 1
)

echo Development API is reachable.
exit /b 0

:check_fcm_prerequisites
echo Checking FCM prerequisites...
if not exist "%PROJECT_DIR%\android\app\google-services.json" (
  echo Missing Firebase Android config: "%PROJECT_DIR%\android\app\google-services.json"
  exit /b 1
)

adb -s %DEVICE_ID% shell pm path com.google.android.gms >nul 2>nul
if errorlevel 1 (
  echo Google Play Services is missing on this emulator. Use a Google APIs / Google Play tablet AVD for FCM.
  exit /b 1
)

echo FCM client prerequisites are ready.
echo Backend FCM send also requires FIREBASE_CLOUD_MESSAGING_ENABLED=true and Firebase service-account credentials.
exit /b 0

:find_tablet
set "DEVICE_ID="

set "ADB_DEVICES_FILE=%TEMP%\eoffice_adb_devices.txt"
adb devices > "%ADB_DEVICES_FILE%" 2>nul

for /f "usebackq skip=1 tokens=1,2" %%D in ("%ADB_DEVICES_FILE%") do (
  if "%%E"=="device" (
    set "AVD_NAME_FILE=%TEMP%\eoffice_avd_name_%%D.txt"
    adb -s %%D exec-out getprop ro.boot.qemu.avd_name > "!AVD_NAME_FILE!" 2>nul
    set "FOUND_AVD_NAME="
    for /f "usebackq delims=" %%N in ("!AVD_NAME_FILE!") do (
      set "FOUND_AVD_NAME=%%N"
    )
    del /q "!AVD_NAME_FILE!" >nul 2>nul
    if "!FOUND_AVD_NAME!"=="%AVD_NAME%" (
      set "DEVICE_ID=%%D"
      del /q "%ADB_DEVICES_FILE%" >nul 2>nul
      exit /b 0
    )
  )
)

del /q "%ADB_DEVICES_FILE%" >nul 2>nul
exit /b 1
