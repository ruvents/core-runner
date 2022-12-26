<?php

use Runner\Messages\JobRequest;
use Runner\Dispatcher;

require_once "vendor/autoload.php";
require_once "GPBMetadata/Messages.php";
require_once "Runner/Messages/JobRequest.php";
require_once "Runner/Dispatcher.php";

(new Dispatcher())->run(
    static function (string $msg): string {
        // Десериализация сообщения в нужный объект. В данном случае, это
        // запрос на выполнение фоновой задачи.
        $req = new JobRequest(); 
        $req->mergeFromString($msg);

        // Выполнение задачи.
        file_put_contents('php://stderr', "Выполнение фоновой задачи {$req->getName()}...\n");
        file_put_contents('php://stderr', "Данные: {$req->getPayload()}\n");
        sleep(2);
        file_put_contents('php://stderr', "Фоновая задача выполнена...\n");

        return 'ok';
    }
);

?>
