ARG TEST_MONGODB_IMAGE=mongo:4.4
FROM ${TEST_MONGODB_IMAGE}
USER root
COPY docker/secret/keyfile /opt/keyfile
RUN chown mongodb /opt/keyfile && chmod 400 /opt/keyfile && mkdir -p /home/mongodb/ && chown mongodb /home/mongodb
RUN mkdir /opt/backups && touch /opt/backups/.gitkeep && chown mongodb /opt/backups
USER mongodb
