#!/bin/sh

#(over)simple coloured test output, from: http://stackoverflow.com/a/27245610

go test -v . | sed "s|PASS|$(printf '\033[32mPASS\033[0m')|g" | sed "s|FAIL|$(printf '\033[31mFAIL\033[0m')|g"
