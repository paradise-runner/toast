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

rem Keep tree-sitter syntax highlighting enabled on Windows by building with cgo.
if not defined CGO_ENABLED set "CGO_ENABLED=1"
if "%CGO_ENABLED%"=="1" (
    if not defined CC (
        where gcc >nul 2>nul
        if not errorlevel 1 set "CC=gcc"
    )
    if not defined CC (
        where clang >nul 2>nul
        if not errorlevel 1 set "CC=clang"
    )
    if not defined CC (
        echo error: CGO_ENABLED=1 requires a Windows C compiler such as w64devkit/MinGW gcc or LLVM clang. 1>&2
        echo        Install one and add it to PATH, or set CGO_ENABLED=0 for a plain-text fallback. 1>&2
        exit /b 1
    )
)

if not exist "bin" mkdir "bin"
"%GO_EXE%" build -o "bin\toast.exe" ".\cmd\toast"
if errorlevel 1 exit /b %errorlevel%

echo Built bin\toast.exe
