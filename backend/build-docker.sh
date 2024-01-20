#!/bin/bash

cd src
go build
mkdir -p out
cp deckyfileserver out/backend
