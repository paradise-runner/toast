@echo off
setlocal

set "GO_EXE=go"
where go >nul 2>nul
if errorlevel 1 (
    if exist "C:\bin\Go\bin\go.exe" (
        set "GO_EXE=C:\bin\Go\bin\go.exe"
    ) else (
        echo error: Go was not found on PATH or at C:\bin\Go\bin\go.exe 1>&2
        exit /b 1
    )
)

if not exist "bin" mkdir "bin"
"%GO_EXE%" build -o "bin\toast.exe" ".\cmd\toast"
if errorlevel 1 exit /b %errorlevel%

echo Built bin\toast.exe
