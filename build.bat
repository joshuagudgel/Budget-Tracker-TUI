@REM filepath: C:\Projects\TUI\budget-tracker-tui\build.bat
@echo off
echo Building for multiple platforms...

if not exist dist mkdir dist

echo Building for Windows...
set GOOS=windows
set GOARCH=amd64
go build -o dist\finances-wrapped-windows.exe .

echo Building for Linux...
set GOOS=linux
set GOARCH=amd64
go build -o dist\finances-wrapped-linux .

echo Building for macOS Intel...
set GOOS=darwin
set GOARCH=amd64
go build -o dist\finances-wrapped-macos-intel .

echo Building for macOS ARM...
set GOOS=darwin
set GOARCH=arm64
go build -o dist\finances-wrapped-macos-arm .

echo Done! Executables are in the dist folder.
pause