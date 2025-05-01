#! /env/sh

mongohost=`getent hosts ${MONGO_HOST} | awk '{ print $1 }'`
kerberos_host=`getent hosts ${KERBEROS_HOST} | awk '{ print $1 }'`

cat > /krb5/krb5.conf <<EOL
[libdefaults]
    default_realm = PERCONATEST.COM
    forwardable = true
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ignore_acceptor_hostname = true
    rdns = false
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
kadmin.local -q "addprinc -pw password1 pmm-test"
kadmin.local -q "ktadd -k /krb5/mongodb.keytab mongodb/${mongohost}@PERCONATEST.COM"
krb5kdc -n
