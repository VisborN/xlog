#!/bin/bash

if [ "$1" = "" ]; then
    go test -v . | sed ''/PASS/s//$(printf "\033[32mPASS\033[0m")/'' | sed ''/FAIL/s//$(printf "\033[31mFAIL\033[0m")/'' | sed ''/RUN/s//$(printf "\033[35mRUN\033[0m")/'' | sed ''/SKIP/s//$(printf "\033[33mSKIP\033[0m")/''
else
    go test -v $1 | sed ''/PASS/s//$(printf "\033[32mPASS\033[0m")/'' | sed ''/FAIL/s//$(printf "\033[31mFAIL\033[0m")/'' | sed ''/RUN/s//$(printf "\033[35mRUN\033[0m")/'' | sed ''/SKIP/s//$(printf "\033[33mSKIP\033[0m")/''
fi
