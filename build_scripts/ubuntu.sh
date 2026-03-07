VERSION=$1
if [ -z "$VERSION" ]; then
    echo "Cannot build - need version number argument, i.e.:"
    echo "./ubuntu.sh v1.2.3"
    exit 1
fi
cd ..
mkdir tmp
go build -o tmp/ha_network_failover main.go
cp config.example.json tmp/
mkdir tmp/storage
cd tmp
tar -czvf ../ubuntu-${VERSION}.tar.gz .
cd ..
rm -rf tmp/
