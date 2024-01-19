#!/bin/bash

cd src
go build

mkdir -p ../bin

cp --preserve=mode deckyfileserver ../bin/backend
