# Usando GitHub Actions

Este repositorio construye y despliega la imagen Docker en [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) en
cada pull request aprobado o en tu propio fork una vez configurado.

## Configurando GitHub Actions

En la configuración del repositorio, bajo secrets, agrega los siguientes secretos:

- `REGISTRY_LOGIN_SERVER`: El registro Docker a usar (ej. `docker.io`).
- `REGISTRY_USERNAME`: El nombre de usuario para iniciar sesión en el registro (ej. `dunglas`).
- `REGISTRY_PASSWORD`: La contraseña para iniciar sesión en el registro (ej. una clave de acceso).
- `IMAGE_NAME`: El nombre de la imagen (ej. `dunglas/frankenphp`).

## Construyendo y Subiendo la Imagen

1. Crea un Pull Request o haz push a tu fork.
2. GitHub Actions construirá la imagen y ejecutará cualquier prueba.
3. Si la construcción es exitosa, la imagen será subida al registro usando la etiqueta `pr-x`, donde `x` es el número del PR.

## Desplegando la Imagen

1. Una vez que el Pull Request sea fusionado, GitHub Actions ejecutará nuevamente las pruebas y construirá una nueva imagen.
2. Si la construcción es exitosa, la etiqueta `main` será actualizada en el registro Docker.

## Lanzamientos (Releases)

1. Crea una nueva etiqueta (tag) en el repositorio.
2. GitHub Actions construirá la imagen y ejecutará cualquier prueba.
3. Si la construcción es exitosa, la imagen será subida al registro usando el nombre de la etiqueta como etiqueta (ej. se crearán `v1.2.3` y `v1.2`).
4. La etiqueta `latest` también será actualizada.
