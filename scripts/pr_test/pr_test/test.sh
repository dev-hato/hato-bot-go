#!/usr/bin/env bash
set -e

cp .env.example .env
go test ./...
