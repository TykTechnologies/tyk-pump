#!/bin/bash
REDIS_PORT=6379
REDIS_HOST="localhost"
REDIS_PASSWORD=""
MONGO_URL="mongodb://localhost/tyk_analytics"
SQL_CONNECTION_STRING=""
SQL_TYPE=""

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
    -postgres=*|--postgres=*)
    SQL_CONNECTION_STRING="${i#*=}"
    SQL_TYPE="postgres"
    shift # past argument=value
    ;;
    -sqlite=*|--sqlite=*)
    SQL_CONNECTION_STRING="${i#*=}"
    SQL_TYPE="sqlite"
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

TEMPLATE_FILE="pump.template.conf"



if [ -z "$SQL_CONNECTION_STRING" ]
then
  echo "Use Mongo   = ${USE_MONGO}"
  echo "Mongo URL   = ${MONGO_URL}"
else
  TEMPLATE_FILE="pumpsql.template.conf"
  echo "Use SQL   = ${SQL_TYPE}"
  echo "SQL Connection string   = ${SQL_CONNECTION_STRING}"
fi
# Set up the editing file

cp $DIR/data/$TEMPLATE_FILE $DIR/pump.conf

# Update variables
sed -i 's/REDIS_HOST/'$REDIS_HOST'/g' $DIR/pump.conf
sed -i 's/REDIS_PORT/'$REDIS_PORT'/g' $DIR/pump.conf
sed -i 's/REDIS_PASSWORD/'$REDIS_PASSWORD'/g' $DIR/pump.conf

if [ -z "$SQL_CONNECTION_STRING" ]
then
  sed -i 's#MONGO_URL#'$MONGO_URL'#g' $DIR/pump.conf
else
  sed -i 's#SQL_TYPE#'$SQL_TYPE'#g' $DIR/pump.conf
  sed -i 's#SQL_CONNECTION_STRING#'$SQL_CONNECTION_STRING'#g' $DIR/pump.conf
fi

echo "==> File written to ./pump.conf"
sudo cp $DIR/pump.conf $DIR/../pump.conf
echo "==> File copied to $DIR/../pump.conf"