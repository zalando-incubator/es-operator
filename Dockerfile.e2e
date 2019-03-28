FROM registry.opensource.zalan.do/stups/alpine:latest
MAINTAINER Team Poirot @ Zalando SE <team-poirot@zalando.de>

# add binary
ADD build/linux/e2e /

ENTRYPOINT ["/e2e", "-test.v", "-test.parallel", "64"]
