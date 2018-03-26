FROM golang:1.10.0-stretch

ENV GOPATH=/go \
    PATH=$GOPATH/bin:$PATH

RUN go get github.com/Masterminds/glide

WORKDIR $GOPATH/src/github.com/myhelix/terracanary

COPY glide.* ./
RUN glide install

COPY . .
RUN go install

