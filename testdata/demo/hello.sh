#!/bin/sh
echo "hello from mini-docker"
echo "hostname: $(hostname)"
echo "pid: $$"
echo "root entries: $(ls / | tr '\n' ' ')"