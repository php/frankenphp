# Usando GitHub Actions

Este repositório constrói e implementa a imagem do Docker no
[Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) em cada pull request
aprovado ou em seu próprio fork após a configuração.

## Configurando GitHub Actions

Nas configurações do repositório, em "Secrets", adicione os seguintes segredos:

- `REGISTRY_LOGIN_SERVER`: O registro do Docker a ser usado (por exemplo,
  `docker.io`).
- `REGISTRY_USERNAME`: O nome de usuário a ser usado para fazer login no
  registro (por exemplo, `dunglas`).
- `REGISTRY_PASSWORD`: A senha a ser usada para fazer login no registro (por
  exemplo, uma chave de acesso).
- `IMAGE_NAME`: O nome da imagem (por exemplo, `dunglas/frankenphp`).

## Construindo e enviando a imagem

1. Crie um pull request ou faça o push para o seu fork.
2. O GitHub Actions construirá a imagem e executará os testes.
3. Se a construção for bem-sucedida, a imagem será enviada para o registro
   usando a tag `pr-x`, onde `x` é o número do PR.

## Implantando a imagem

1. Após o merge do pull request, o GitHub Actions executará os testes novamente
   e criará uma nova imagem.
2. Se a construção for bem-sucedida, a tag `main` será atualizada no registro do
   Docker.

## Versões

1. Crie uma nova tag no repositório.
2. O GitHub Actions construirá a imagem e executará os testes.
3. Se a construção for bem-sucedida, a imagem será enviada para o registro
   usando o nome da tag como tag (por exemplo, `v1.2.3` e `v1.2` serão criadas).
4. A tag `latest` também será atualizada.
