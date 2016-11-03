FROM golang:latest

MAINTAINER Dominika Hodovska <dhodovsk@redhat.com>

EXPOSE 8081
COPY cluster-capacity /cluster-capacity
COPY config/default-scheduler.yaml /config/default-scheduler.yaml
ENTRYPOINT ["/cluster-capacity"]
CMD ["--help"]
