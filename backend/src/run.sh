#!/bin/bash

npx nodemon --ext "go,html" --signal SIGTERM --exec go run *.go -- -f $HOME -verbose
