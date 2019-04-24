#!/bin/sh

APPNAME=`basename ${PWD}`
for OS in windows linux darwin; do
  mkdir -p ${OS}
  GOOS=${OS} GOARCH=amd64 go build -o ${OS}/${APPNAME}
  if [ ${OS} == "windows" ]; then
    mv windows/${APPNAME} windows/${APPNAME}.exe
  fi
done
