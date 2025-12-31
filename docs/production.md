# Deploying in Production

In this tutorial, we will learn how to deploy a PHP application on a single server using Docker Compose.

If you're using Symfony, prefer reading the "[Deploy in production](https://github.com/dunglas/symfony-docker/blob/main/docs/production.md)" documentation entry of the Symfony Docker project (which uses FrankenPHP).

If you're using API Platform (which also uses FrankenPHP), refer to [the deployment documentation of the framework](https://api-platform.com/docs/deployment/).

## Preparing Your App

First, create a `Dockerfile` in the root directory of your PHP project:

```dockerfile
FROM dunglas/frankenphp

# Be sure to replace "your-domain-name.example.com" by your domain name
ENV SERVER_NAME=your-domain-name.example.com
# If you want to disable HTTPS, use this value instead:
#ENV SERVER_NAME=:80

# If your project is not using the "public" directory as the web root, you can set it here:
# ENV SERVER_ROOT=web/

# Enable PHP production settings
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

# Copy the PHP files of your project in the public directory
COPY . /app/public
# If you use Symfony or Laravel, you need to copy the whole project instead:
#COPY . /app
```

Refer to "[Building Custom Docker Image](docker.md)" for more details and options,
and to learn how to customize the configuration, install PHP extensions and Caddy modules.

If your project uses Composer,
be sure to include it in the Docker image and to install your dependencies.

Then, add a `compose.yaml` file:

```yaml
services:
  php:
    image: dunglas/frankenphp
    restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - caddy_data:/data
      - caddy_config:/config

# Volumes needed for Caddy certificates and configuration
volumes:
  caddy_data:
  caddy_config:
```

> [!NOTE]
>
> The previous examples are intended for production usage.
> In development, you may want to use a volume, a different PHP configuration and a different value for the `SERVER_NAME` environment variable.
>
> Take a look to the [Symfony Docker](https://github.com/dunglas/symfony-docker) project
> (which uses FrankenPHP) for a more advanced example using multi-stage images,
> Composer, extra PHP extensions, etc.

Finally, if you use Git, commit these files and push.

## Preparing a Server

To deploy your application in production, you need a server.
In this tutorial, we will use a virtual machine provided by DigitalOcean, but any Linux server can work.
If you already have a Linux server with Docker installed, you can skip straight to [the next section](#configuring-a-domain-name).

Otherwise, use [this affiliate link](https://m.do.co/c/5d8aabe3ab80) to get $200 of free credit, create an account, then click on "Create a Droplet".
Then, click on the "Marketplace" tab under the "Choose an image" section and search for the app named "Docker".
This will provision an Ubuntu server with the latest versions of Docker and Docker Compose already installed!

For test purposes, the cheapest plans will be enough.
For real production usage, you'll probably want to pick a plan in the "general purpose" section to fit your needs.

![Deploying FrankenPHP on DigitalOcean with Docker](digitalocean-droplet.png)

You can keep the defaults for other settings, or tweak them according to your needs.
Don't forget to add your SSH key or create a password then press the "Finalize and create" button.

Then, wait a few seconds while your Droplet is provisioning.
When your Droplet is ready, use SSH to connect:

```console
ssh root@<droplet-ip>
```

## Configuring a Domain Name

In most cases, you'll want to associate a domain name with your site.
If you don't own a domain name yet, you'll have to buy one through a registrar.

Then create a DNS record of type `A` for your domain name pointing to the IP address of your server:

```dns
your-domain-name.example.com.  IN  A     207.154.233.113
```

Example with the DigitalOcean Domains service ("Networking" > "Domains"):

![Configuring DNS on DigitalOcean](digitalocean-dns.png)

> [!NOTE]
>
> Let's Encrypt, the service used by default by FrankenPHP to automatically generate a TLS certificate doesn't support using bare IP addresses. Using a domain name is mandatory to use Let's Encrypt.

## Deploying

Copy your project on the server using `git clone`, `scp`, or any other tool that may fit your need.
If you use GitHub, you may want to use [a deploy key](https://docs.github.com/en/free-pro-team@latest/developers/overview/managing-deploy-keys#deploy-keys).
Deploy keys are also [supported by GitLab](https://docs.gitlab.com/ee/user/project/deploy_keys/).

Example with Git:

```console
git clone git@github.com:<username>/<project-name>.git
```

Go into the directory containing your project (`<project-name>`), and start the app in production mode:

```console
docker compose up --wait
```

Your server is up and running, and an HTTPS certificate has been automatically generated for you.
Go to `https://your-domain-name.example.com` and enjoy!

> [!CAUTION]
>
> Docker can have a cache layer, make sure you have the right build for each deployment or rebuild your project with `--no-cache` option to avoid cache issue.

## Deploying on Multiple Nodes

If you want to deploy your app on a cluster of machines, you can use [Docker Swarm](https://docs.docker.com/engine/swarm/stack-deploy/),
which is compatible with the provided Compose files.
To deploy on Kubernetes, take a look at [the Helm chart provided with API Platform](https://api-platform.com/docs/deployment/kubernetes/), which uses FrankenPHP.
