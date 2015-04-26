# Example image to test / document a simple installation

FROM ubuntu:14.04

MAINTAINER Daniel Danilatos

RUN apt-get update
RUN apt-get install -y wget git build-essential libssl-dev

COPY ./deps/go1.4.2.linux-amd64.tar.gz /tmp/
COPY ./deps/haproxy-1.5.3.tar.gz /tmp/
ADD ./install-deps.sh /tmp/install-deps.sh
RUN /bin/sh /tmp/install-deps.sh

ADD ./install-camus.sh /tmp/install-camus.sh
RUN adduser --disabled-password --gecos "" myapp
RUN /bin/sh /tmp/install-camus.sh myapp

CMD su -l myapp -c ./start-camus.sh

