#!/bin/bash

npx nodemon --watch "./**/*.go" --signal SIGTERM --exec go run *.go -- -f $HOME -verbose
