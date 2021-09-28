# Installing Dragonfly Manager Server

This topic explains how to install the Dragonfly manager server.

## Context

Install manager in one of the following ways:

- Deploying with Docker: Recommended for production usage.
- Deploying with physical machines.

## Prerequisites

When deploying with Docker, the following conditions must be met.

Required Software | Version Limit
---|---
Git|1.9.1+
Docker|1.12.0+

When deploying with physical machines, the following conditions must be met.

Required Software | Version Limit
---|---
Git|1.9.1+
Golang|1.12.x
Nginx|0.8+

## Procedure - When Deploying with Docker

### Get manager image

You can get it from [DockerHub](https://hub.docker.com/) directly.

1. Obtain the latest Docker image of the manager.

```sh
docker pull d7yio/manager
```

Or you can build your own manager image.

1. Obtain the source code of Dragonfly.

```sh
git clone https://github.com/dragonflyoss/Dragonfly2.git
```

2. Enter the project directory.

```sh
cd Dragonfly2
```

3. Build the Docker image.

```sh
TAG="2.0.0"
make docker-build-manager D7Y_VERSION=$TAG
```

4. Obtain the latest Docker image ID of the manager.

```sh
docker image ls | grep 'manager' | awk '{print $3}' | head -n1
```

### Start manager

**NOTE:** Replace ${managerDockerImageId} with the ID obtained at the previous step.

```sh
docker run -d --name manager --restart=always -p 8080:8080 -p 65003:65003 ${managerDockerImageId}
```

After manager is installed, run the following commands to verify if **manager** is started, and if Port `8080` and `65003` is available.

```sh
telnet 127.0.0.1 8080
telnet 127.0.0.1 65003
```

### Manager console

Now you can open brower and visit [console](http://localhost:8080)

Console features preview reference document [console preview](../../user-guide/console/preview.md).

## Procedure - When Deploying with Physical Machines

### Get cdn executable file

1. Download a binary package of the manager. You can download one of the latest builds for Dragonfly on the [github releases page](https://github.
   com/dragonflyoss/Dragonfly2/releases).

```sh
version=2.0.0
wget https://github.com/dragonflyoss/Dragonfly2/releases/download/v$version/Dragonfly2_$version_linux_amd64.tar.gz
```

2. Unzip the package.

```bash
# Replace `xxx` with the installation directory.
tar -zxf Dragonfly2_2.0.0_linux_amd64.tar.gz -C xxx
```

3. Move the `manager` to your `PATH` environment variable to make sure you can directly use `manager` command.

Or you can build your own manager executable file.

1. Obtain the source code of Dragonfly.

```sh
git clone https://github.com/dragonflyoss/Dragonfly2.git
```

2. Enter the project directory.

```sh
cd Dragonfly2
```

3. Compile the source code.

```sh
make build-manager && make install-manager
```

### Start manager

```sh
manager --options
```

After manager is installed, run the following commands to verify if **manager** is started, and if Port `8080` and `65003` is available.

```sh
telnet 127.0.0.1 8080
telnet 127.0.0.1 65003
```

### Manager console

Now you can open brower and visit [console](http://localhost:8080)

Console features preview reference document [console preview](../../user-guide/console/preview.md).
