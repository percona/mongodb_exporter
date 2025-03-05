#! /env/sh

mongohost=`getent hosts ${MONGO_HOST} | awk '{ print $1 }'`
kerberos_host=`getent hosts ${KERBEROS_HOST} | awk '{ print $1 }'`
gateway_ip=`ip route | grep default | awk '{print $3}'`

cat > /etc/krb5.conf <<EOL
[libdefaults]
    default_realm = PERCONATEST.COM
    forwardable = true
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ignore_acceptor_hostname = true
    rdns = false
    noaddresses = TRUE
[realms]
    PERCONATEST.COM = {
        kdc_ports = 88
        kdc = $kerberos_host
    }
[domain_realm]
    .perconatest.com = PERCONATEST.COM
    perconatest.com = PERCONATEST.COM
    $kerberos_host = PERCONATEST.COM
EOL

kdb5_util create -s -P password
kadmin.local -q "addprinc -pw password root/admin"
kadmin.local -q "addprinc -pw mongodb mongodb/${mongohost}"
kadmin.local -q "addprinc -pw mongodb mongodb/${gateway_ip}"
kadmin.local -q "addprinc -pw password1 pmm-test"

kadmin.local -q "ktadd -k /tmp/mongodb.keytab mongodb/${mongohost}@PERCONATEST.COM"
kadmin.local -q "ktadd -k /tmp/exporter.keytab mongodb/${gateway_ip}@PERCONATEST.COM"

kadmin.local -q "ktadd -k /tmp/mongodb.keytab pmm-test@PERCONATEST.COM"
kadmin.local -q "ktadd -k /tmp/exporter.keytab pmm-test@PERCONATEST.COM"

krb5kdc -n