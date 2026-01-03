# Despliegue en Producción

En este tutorial, aprenderemos cómo desplegar una aplicación PHP en un único servidor usando Docker Compose.

Si estás usando Symfony, consulta la documentación "[Despliegue en producción](https://github.com/dunglas/symfony-docker/blob/main/docs/production.md)" del proyecto Symfony Docker (que usa FrankenPHP).

Si estás usando API Platform (que también usa FrankenPHP), consulta [la documentación de despliegue del framework](https://api-platform.com/docs/deployment/).

## Preparando tu Aplicación

Primero, crea un archivo `Dockerfile` en el directorio raíz de tu proyecto PHP:

```dockerfile
FROM dunglas/frankenphp

# Asegúrate de reemplazar "tu-dominio.ejemplo.com" por tu nombre de dominio
ENV SERVER_NAME=tu-dominio.ejemplo.com
# Si quieres deshabilitar HTTPS, usa este valor en su lugar:
#ENV SERVER_NAME=:80

# Si tu proyecto no usa el directorio "public" como raíz web, puedes establecerlo aquí:
# ENV SERVER_ROOT=web/

# Habilitar configuración de producción de PHP
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

# Copiar los archivos PHP de tu proyecto en el directorio público
COPY . /app/public
# Si usas Symfony o Laravel, necesitas copiar todo el proyecto en su lugar:
#COPY . /app
```

Consulta "[Construyendo una Imagen Docker Personalizada](docker.md)" para más detalles y opciones,
y para aprender cómo personalizar la configuración, instalar extensiones PHP y módulos de Caddy.

Si tu proyecto usa Composer,
asegúrate de incluirlo en la imagen Docker e instalar tus dependencias.

Luego, agrega un archivo `compose.yaml`:

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

# Volúmenes necesarios para los certificados y configuración de Caddy
volumes:
  caddy_data:
  caddy_config:
```

> [!NOTE]
>
> Los ejemplos anteriores están destinados a uso en producción.
> En desarrollo, es posible que desees usar un volumen, una configuración PHP diferente y un valor diferente para la variable de entorno `SERVER_NAME`.
>
> Consulta el proyecto [Symfony Docker](https://github.com/dunglas/symfony-docker)
> (que usa FrankenPHP) para un ejemplo más avanzado usando imágenes multi-etapa,
> Composer, extensiones PHP adicionales, etc.

Finalmente, si usas Git, haz commit de estos archivos y haz push.

## Preparando un Servidor

Para desplegar tu aplicación en producción, necesitas un servidor.
En este tutorial, usaremos una máquina virtual proporcionada por DigitalOcean, pero cualquier servidor Linux puede funcionar.
Si ya tienes un servidor Linux con Docker instalado, puedes saltar directamente a [la siguiente sección](#configurando-un-nombre-de-dominio).

De lo contrario, usa [este enlace de afiliado](https://m.do.co/c/5d8aabe3ab80) para obtener $200 de crédito gratuito, crea una cuenta y luego haz clic en "Crear un Droplet".
Luego, haz clic en la pestaña "Marketplace" bajo la sección "Elegir una imagen" y busca la aplicación llamada "Docker".
Esto aprovisionará un servidor Ubuntu con las últimas versiones de Docker y Docker Compose ya instaladas.

Para fines de prueba, los planes más económicos serán suficientes.
Para un uso real en producción, probablemente querrás elegir un plan en la sección "uso general" que se adapte a tus necesidades.

![Desplegando FrankenPHP en DigitalOcean con Docker](digitalocean-droplet.png)

Puedes mantener los valores predeterminados para otras configuraciones o ajustarlos según tus necesidades.
No olvides agregar tu clave SSH o crear una contraseña y luego presionar el botón "Finalizar y crear".

Luego, espera unos segundos mientras se aprovisiona tu Droplet.
Cuando tu Droplet esté listo, usa SSH para conectarte:

```console
ssh root@<ip-del-droplet>
```

## Configurando un Nombre de Dominio

En la mayoría de los casos, querrás asociar un nombre de dominio a tu sitio.
Si aún no tienes un nombre de dominio, deberás comprar uno a través de un registrador.

Luego, crea un registro DNS de tipo `A` para tu nombre de dominio que apunte a la dirección IP de tu servidor:

```dns
tu-dominio.ejemplo.com.  IN  A     207.154.233.113
```

Ejemplo con el servicio de Dominios de DigitalOcean ("Redes" > "Dominios"):

![Configurando DNS en DigitalOcean](../digitalocean-dns.png)

> [!NOTE]
>
> Let's Encrypt, el servicio utilizado por defecto por FrankenPHP para generar automáticamente un certificado TLS, no soporta el uso de direcciones IP puras. Usar un nombre de dominio es obligatorio para usar Let's Encrypt.

## Despliegue

Copia tu proyecto en el servidor usando `git clone`, `scp` o cualquier otra herramienta que se ajuste a tus necesidades.
Si usas GitHub, es posible que desees usar [una clave de despliegue](https://docs.github.com/es/free-pro-team@latest/developers/overview/managing-deploy-keys#deploy-keys).
Las claves de despliegue también son [soportadas por GitLab](https://docs.gitlab.com/ee/user/project/deploy_keys/).

Ejemplo con Git:

```console
git clone git@github.com:<usuario>/<nombre-proyecto>.git
```

Ve al directorio que contiene tu proyecto (`<nombre-proyecto>`) e inicia la aplicación en modo producción:

```console
docker compose up --wait
```

Tu servidor está en funcionamiento y se ha generado automáticamente un certificado HTTPS para ti.
Ve a `https://tu-dominio.ejemplo.com` y ¡disfruta!

> [!CAUTION]
>
> Docker puede tener una capa de caché, asegúrate de tener la compilación correcta para cada despliegue o vuelve a compilar tu proyecto con la opción `--no-cache` para evitar problemas de caché.

## Despliegue en Múltiples Nodos

Si deseas desplegar tu aplicación en un clúster de máquinas, puedes usar [Docker Swarm](https://docs.docker.com/engine/swarm/stack-deploy/),
que es compatible con los archivos Compose proporcionados.
Para desplegar en Kubernetes, consulta [el gráfico Helm proporcionado con API Platform](https://api-platform.com/docs/deployment/kubernetes/), que usa FrankenPHP.
