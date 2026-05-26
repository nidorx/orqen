@echo off
REM code_stats.bat - Count files by extension in a directory
REM Usage: code_stats.bat [dir]
REM Note: Uses dir + manual counter to avoid find.exe conflicts with Git Bash

setlocal enabledelayedexpansion

set "DIR=%~1"
if "%DIR%"=="" set "DIR=."

echo === Code Stats ===
echo Directory: %DIR%
echo.

for %%E in (go ts tsx js py rs toml yaml yml md sql sh bat) do (
  set /a count=0
  for /f "delims=" %%F in ('dir /s /b /a-d "%DIR%\*.%%E" 2^>nul') do set /a count+=1
  if !count! gtr 0 echo   .%%~E    !count! file(s^)
)

echo   ------  -----------

set /a total=0
for /f "delims=" %%F in ('dir /s /b /a-d "%DIR%" 2^>nul') do set /a total+=1
echo   total   %total% file(s^)
