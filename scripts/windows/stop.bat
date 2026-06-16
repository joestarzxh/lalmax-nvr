@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0nvr.ps1" stop %*
exit /b %ERRORLEVEL%
