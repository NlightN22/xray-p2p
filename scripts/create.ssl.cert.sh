opkg update
opkg install openssl-util
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout /etc/xray/key.pem -out /etc/xray/cert.pem -subj '/CN=domain.tld'
chmod 600 /etc/xray/key.pem /etc/xray/cert.pem