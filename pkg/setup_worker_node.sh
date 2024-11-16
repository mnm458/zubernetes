#!/bin/bash

sudo apt-get update
sudo apt-get install -y docker.io docker-compose

sudo systemctl start docker
sudo systemctl start docker
sudo systemctl enable docker

mkdir -p /opt/cadvisor

cat << EOF > /opt/cadvisor/docker-compose.yml
version: '3.2'
services:
  cadvisor:
    image: gcr.io/cadvisor/cadvisor:latest
    container_name: cadvisor
    ports:
    - 8080:8080
    volumes:
    - /:/rootfs:ro
    - /var/run:/var/run:rw
    - /sys:/sys:ro
    - /var/lib/docker/:/var/lib/docker:ro
    depends_on:
    - redis
  redis:
    image: redis:latest
    container_name: redis
    ports:
    - 6379:6379
EOF

cd /opt/cadvisor
sudo docker-compose up -d

sudo wget https://zubernetes-worker-binary-url -O /usr/local/bin/worker
sudo chmod +x /usr/local/bin/worker
sudo systemctl start worker
version 