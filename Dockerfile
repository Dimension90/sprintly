FROM nginx:1.27-alpine

COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY dist/client /usr/share/nginx/html

EXPOSE 3000
