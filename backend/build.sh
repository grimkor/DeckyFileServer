#!/bin/bash

cd src
go build
mkdir -p ../../bin

mv deckyfileserver ../../bin/backend
