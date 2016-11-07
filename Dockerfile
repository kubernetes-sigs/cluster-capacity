FROM golang:latest

MAINTAINER Dominika Hodovska <dhodovsk@redhat.com>

EXPOSE 8081
COPY hypercc /bin/hypercc
RUN ln -sf /bin/hypercc /bin/cluster-capacity
RUN ln -sf /bin/hypercc /bin/genpod
COPY config/default-scheduler.yaml /config/default-scheduler.yaml
CMD ["/bin/cluster-capacity --help"]
