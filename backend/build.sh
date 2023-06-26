#!/bin/bash

cargo build

mkdir -p ../bin

cp --preserve=mode ./target/debug/deckyfileserver-rs ../bin/backend
