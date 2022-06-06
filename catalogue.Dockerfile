FROM  ubuntu:18.04
RUN apt update
RUN apt install -y net-tools ca-certificates  bash

WORKDIR /
COPY cataloguesvc /cataloguesvc
COPY images/ /images/
RUN ls -ltr /
ENV GOTRACEBACK=single

EXPOSE 80

CMD ["/cataloguesvc"]
