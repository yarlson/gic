#!/bin/sh
set -eu

target=${1:-.}

gremlins unleash "$target"
