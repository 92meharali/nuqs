# Builds the public FTP handshake fixture used in the lab.
FROM alpine:3.23@sha256:469c9ea410b9215eaee7976ba6c0639039b444be8e5d638f7abbada189971eb2
RUN apk add --no-cache vsftpd=3.0.5-r3 && mkdir -p /srv/public /var/empty && chmod 0555 /srv/public /var/empty
COPY vsftpd.conf /etc/vsftpd/test.conf
ENTRYPOINT ["/usr/sbin/vsftpd", "/etc/vsftpd/test.conf"]
