#!/usr/bin/env bash
set -e

cp .env.example .env
export TAG_NAME="${HEAD_REF//\//-}"
dockle_version="$(cat .dockle-version)"
curl -L -o dockle.deb "https://github.com/goodwithtech/dockle/releases/download/v${dockle_version}/dockle_${dockle_version}_Linux-64bit.deb"
sudo dpkg -i dockle.deb

for filename in misskey.docker-compose.yml mixi2.docker-compose.yml; do
  docker compose -f docker-compose.yml -f "$filename" pull
  docker compose -f docker-compose.yml -f "$filename" up -d
  for image_name in $(docker compose -f docker-compose.yml -f "$filename" images | awk 'OFS=":" {print $2,$3}' | tail -n +2); do
    cmd="dockle --exit-code 1 ${image_name}"
    echo "> ${cmd}"
    eval "${cmd}"
  done
done
