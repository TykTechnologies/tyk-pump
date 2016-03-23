#!/bin/bash
REDIS_PORT=6379
REDIS_HOST="localhost"
REDIS_PASSWORD=""
MONGO_URL="mongodb://localhost/tyk_analytics"

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

for i in "$@"
do
case $i in
    -r=*|--redishost=*)
    REDIS_HOST="${i#*=}"
    shift # past argument=value
    ;;
    -p=*|--redisport=*)
    REDIS_PORT="${i#*=}"
    shift # past argument=value
    ;;
    -s=*|--redispass=*)
    REDIS_PASSWORD="${i#*=}"
    shift # past argument=value
    ;;
    -m=*|--mongo=*)
    MONGO_URL="${i#*=}"
    shift # past argument=value
    ;;
    --default)
    DEFAULT=YES
    shift # past argument with no value
    ;;
    *)
            # unknown option
    ;;
esac
done

echo "Redis Host  = ${REDIS_HOST}"
echo "Redis Port  = ${REDIS_PORT}"
echo "Redis PW    = ${REDIS_PASSWORD}"
echo "Use Mongo   = ${USE_MONGO}"
echo "Mongo URL   = ${MONGO_URL}"

# Set up the editing file
TEMPLATE_FILE="pump.template.conf"

cp $DIR/data/$TEMPLATE_FILE $DIR/pump.conf

# Update variables
sed -i 's/REDIS_HOST/'$REDIS_HOST'/g' $DIR/tyk.conf
sed -i 's/REDIS_PORT/'$REDIS_PORT'/g' $DIR/tyk.conf
sed -i 's/REDIS_PASSWORD/'$REDIS_PASSWORD'/g' $DIR/tyk.conf
sed -i 's#MONGO_URL#'$MONGO_URL'#g' $DIR/tyk.conf

echo "==> File written to ./pump.conf"
sudo cp $DIR/pump.conf $DIR/../pump.conf
echo "==> File copied to $DIR/../pump.conf"