#!/usr/bin/env bash

go_version=$(grep golang: Dockerfile | sed -e 's!FROM golang:\([0-9.]*\)-.*!\1!g')
sed -i -e "s/^go \([0-9.]*\)/go $go_version/g" go.mod
