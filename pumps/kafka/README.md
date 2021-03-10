# Kafka Pump

## Kafka Config

* `broker`: The list of brokers used to discover the partitions available on the kafka cluster. E.g. "localhost:9092"
* `use_ssl`: Enables SSL connection. 
* `ssl_insecure_skip_verify`: Controls whether the pump client verifies the kafka server's certificate chain and host name.
* `client_id`: Unique identifier for client connections established with Kafka.
* `topic`: The topic that the writer will produce messages to.
* `timeout`: Timeout is the maximum amount of time will wait for a connect or write to complete. 
* `compressed`: Enable "github.com/golang/snappy" codec to be used to compress Kafka messages. By default is false
* `meta_data`: Can be used to set custom metadata inside the kafka message
* `ssl_cert_file`: Can be used to set custom certificate file for authentication with kafka.
* `ssl_key_file`: Can be used to set custom key file for authentication with kafka.


## Examples
### Common Kafka Pump - No auth.
Pretty standard Kafka pump configuration without any type of authentication. Useful for local testing with Docker and Bitnami.
```.json
    "kafka": {
        "type": "kafka",
        "meta": {
            "broker": [
                "<BROKER_HOST>:<BROKER_PORT>"
            ],
            "topic": "<TOPIC_NAME>",
            "client_id": "<CLIENT_ID>",
            "timeout": 60,
            "compressed": true,
            "meta_data": {
                "key": "value"
            }
        }
    }
```

### Kafka Pump +  Basic Auth (Plain).
In the following example, `sasl_username` it's the username or token / key and `sasl_password` it's the password or secret.
```.json
    "kafka": {
        "type": "kafka",
        "meta": {
            "broker": [
                "<BROKER_HOST>:<BROKER_PORT>"
            ],
            "topic": "<TOPIC_NAME>",
            "client_id": "<CLIENT_ID>",
            "timeout": 60,
            "compressed": true,
            "meta_data": {
                "key": "value"
            },
            "use_ssl":true,
            "sasl_mechanism":"plain",
            "sasl_username":"<USERNAME>",
            "sasl_password":"<PASSWORD>"
        }
    }
```
### Kafka Pump + Basic Auth (Scram).
In the following example, `sasl_username` it's the username or token / key, `sasl_password` it's the password or secret and `sasl_algorithm` It's the algorithm specified for scram mechanism. It could be `sha-512` or `sha-256`, sha-256 it's the default value if the algorithm is not configured.
```.json
    "kafka": {
        "type": "kafka",
        "meta": {
            "broker": [
                "<BROKER_HOST>:<BROKER_PORT>"
            ],
            "topic": "<TOPIC_NAME>",
            "client_id": "<CLIENT_ID>",
            "timeout": 60,
            "compressed": true,
            "meta_data": {
                "key": "value"
            },
            "use_ssl":true,
            "sasl_mechanism":"scram",
            "sasl_algorithm":"sha-512",
            "sasl_username":"<USERNAME>",
            "sasl_password":"<PASSWORD>"
        }
    }
```


### Kafka Pump + Mutual TLS.
In the following example, we set Kafka pump to use mTLS giving the path for the cert and key file.
```.json
    "kafka": {
      "type": "kafka",
      "meta": {
        "broker": [
            ""<BROKER_HOST>:<BROKER_PORT>"
        ],
        "client_id": "<CLIENT_ID>",
        "topic": "<TOPIC>",
        "timeout": 60,
        "compressed": true,
        "meta_data": {
            "key": "value"
        },
        "ssl_insecure_skip_verify":true,
        "use_ssl":true,
        "ssl_cert_file":"<CERT_PATH>",
        "ssl_key_file":"<KEY_PATH>",
      }
    }
```

