# epg-vmedia

Application Go permanente qui récupère le guide TV du fournisseur VMedia/Fizz, le stocke dans SQLite et publie un guide XMLTV de 14 jours. L’application fournit également une interface web pour sélectionner les chaînes et choisir les logos.

## Fonctionnalités

- Guide XMLTV pré-généré sur 14 jours, disponible sur `/epg.xml`.
- Guides dynamiques, par exemple `/epg.xml?days=7`.
- Interface de sélection des chaînes sur `/channels`.
- Logos de chaînes sur `/logos/` et cache des images de programmes sur `/images/`.
- Base SQLite et données conservées dans `/data`.
- Actualisation planifiée du guide, du catalogue des chaînes et du cache d’images.

## Déploiement Docker / Portainer

L’image publique est publiée sur GitHub Container Registry :

`ghcr.io/lucforcier/epg-vmedia:latest`

Le port interne est **5580**. Par défaut, Compose le publie sur le port **18088** de l’hôte.

### 1. Préparer le répertoire persistant

Exemple pour le serveur Debian :

```sh
mkdir -p /raid/portainer/iptv/epg-vmedia
chown -R 10001:10001 /raid/portainer/iptv/epg-vmedia
```

Le conteneur s’exécute avec l’UID/GID `10001:10001` et doit pouvoir écrire dans ce répertoire.

### 2. Créer la stack Portainer

Dans Portainer, crée une Stack nommée `epg-vmedia`, puis colle le contenu de `docker-compose.yml`. Si tu utilises un dépôt Git, sélectionne le dépôt et la branche contenant ces fichiers.

Configure les variables suivantes dans l’environnement de la Stack :

| Variable | Valeur / rôle |
| --- | --- |
| `EPG_HTTP_PORT` | Port publié sur Debian, par défaut `18088` |
| `EPG_BASE_URL` | URL HTTP joignable par les clients IPTV, par exemple `http://192.0.2.10:18088` ; remplace l’adresse d’exemple par celle de ton serveur |
| `EPG_DATA_DIR` | Répertoire hôte persistant, par défaut `./data` |
| `TZ` | Fuseau horaire de planification, par défaut `America/Toronto` |

Pour le chemin de stockage recommandé ci-dessus, définis `EPG_DATA_DIR=/raid/portainer/iptv/epg-vmedia`.

### 3. Premier démarrage

Après démarrage, ouvre :

- Interface de gestion des chaînes : `http://ADRESSE_DU_SERVEUR:18088/channels`
- Guide XMLTV : `http://ADRESSE_DU_SERVEUR:18088/epg.xml`

Au premier démarrage, le catalogue des chaînes doit être importé depuis VMedia, puis les chaînes désirées doivent être activées dans l’interface. Le guide principal ne peut pas être généré tant qu’aucune chaîne n’est activée. Si le guide retourne une erreur ou une réponse 404 au premier démarrage, consulte les journaux de la Stack et termine l’initialisation des chaînes.

### 4. Vérification

```sh
docker compose ps
docker compose logs -f epg-vmedia
curl -I http://127.0.0.1:18088/channels
curl -I http://127.0.0.1:18088/epg.xml
```

Le guide XMLTV principal est un fichier pré-généré : l’accès à `/epg.xml` ne lance pas une récupération du guide auprès de VMedia. Les requêtes avec paramètres, comme `/epg.xml?days=7`, génèrent une réponse dynamique.

## Configuration de planification

Les valeurs par défaut sont les suivantes :

- Génération XMLTV : tous les jours à `03:00`.
- Actualisation des chaînes et logos : le lundi à `03:30`.
- Nettoyage du cache d’images : le lundi à `04:00`.
- Fuseau horaire : `America/Toronto`.

Les heures sont configurables avec les variables `EPG_SCHEDULER_TIME`, `EPG_CHANNEL_REFRESH_TIME` et `IMAGE_CACHE_CLEANUP_TIME`.

## Construction locale

```sh
go test ./...
go vet ./...
go build -o epg-vmedia ./cmd/epg-vmedia
docker build -t epg-vmedia:local .
```

## Publication

Le workflow GitHub Actions exécute les tests et contrôles Go, puis publie sur GHCR les changements poussés sur `main` et les tags `v*`. La première publication peut nécessiter que le propriétaire autorise le package à être public dans les paramètres du package GHCR.

## Licence et services tiers

Ce projet n’est pas affilié à VMedia ou Fizz. L’accès à l’API et l’utilisation des logos/images restent soumis aux conditions du fournisseur. Vérifie les droits et conditions applicables avant toute redistribution publique.
