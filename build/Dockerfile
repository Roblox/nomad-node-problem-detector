FROM ubuntu:20.04

LABEL maintainer="Shishir Mahajan <smahajan@roblox.com>"

COPY ./extract.sh /usr/local/bin/extract.sh
COPY ./nnpd.tar /tmp/nnpd.tar
RUN chmod +x /usr/local/bin/extract.sh

ENTRYPOINT ["extract.sh"]
