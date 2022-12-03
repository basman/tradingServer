#!/bin/bash

export GIN_MODE=release

rm -f stop
while [ ! -e stop ]; do
	./tradingServer >>run.log 2>&1
	sleep 5
done
