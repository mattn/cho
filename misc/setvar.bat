@echo off
if "%2" equ "" goto err
for /f "delims=;" %%i in ('%2 ^| nkf -Sw ^| cho -cl ^| nkf -Ws') do set %1=%%i
goto eof
:err
echo usage: %~0 [VARNAME] [COMMAND]
