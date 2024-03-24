@echo off
set CUR_DIR=%~dp0
pushd %CUR_DIR%js\client
echo updating client...
call npm i
call npm run build
echo updated client.
popd
pushd %CUR_DIR%js\app
echo updating app...
call npm i
echo updated app.
echo start app...
set HOST=0.0.0.0
call npm run start-simple
popd