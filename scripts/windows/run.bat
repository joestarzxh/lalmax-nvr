@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0nvr.ps1" run %*
exit /b %ERRORLEVEL%
