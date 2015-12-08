# ToxStatus
Status page written in Go that keeps track of Tox bootstrap nodes.<br>The entire codebase is licensed under [AGPL](LICENSE) unless stated otherwise.

# Deploying
Using the included Dockerfile in the 'docker' folder:

```
docker build -t toxstatus .
docker run -d --restart=always -p 8081:8081 --name toxstatus toxstatus
```
