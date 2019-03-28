FROM registry.opensource.zalan.do/stups/alpine:latest
MAINTAINER Team Poirot @ Zalando SE <team-poirot@zalando.de>

# add binary
ADD build/linux/es-operator /

ENTRYPOINT ["/es-operator"]
