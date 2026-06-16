@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0nvr.ps1" restart %*
exit /b %ERRORLEVEL%
