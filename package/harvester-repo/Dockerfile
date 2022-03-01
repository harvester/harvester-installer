FROM registry.suse.com/bci/bci-base:15.3

RUN zypper -n rm container-suseconnect && \
    zypper -n in nginx && \
    zypper -n clean -a && rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/* && \
    ln -sf /dev/stdout /var/log/nginx/access.log && \
    ln -sf /dev/stderr /var/log/nginx/error.log

COPY charts /srv/www/htdocs/charts

EXPOSE 80

STOPSIGNAL SIGQUIT

CMD ["nginx", "-g", "daemon off;"]
