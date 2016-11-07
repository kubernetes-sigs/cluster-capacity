FROM golang:latest

MAINTAINER Dominika Hodovska <dhodovsk@redhat.com>

EXPOSE 8081
COPY cluster-capacity /bin/cluster-capacity
COPY genpod /bin/genpod
COPY config/default-scheduler.yaml /config/default-scheduler.yaml
CMD ["/bin/cluster-capacity --help"]
