@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0nvr.ps1" test %*
exit /b %ERRORLEVEL%
