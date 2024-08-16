<?php

/**
 * Простой скрипт для тригера рассылок сообщений по WS через pubsub механизм
 * Redis. Вызывай с запущенным сервером.
 */

$redis = new \Redis([
    'connectTimeout' => 5,
    'retryInterval' => 5,
    'readTimeout' => 5,
]);

$redis->connect('tcp://localhost:6379');
$redis->publish('chat:64239268-dc82-4c12-a22e-42d568c5b137', 'new message!');
$redis->close();

?>
