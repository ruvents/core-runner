<?php

require_once "vendor/autoload.php";
require_once "Request.php";
require_once "Response.php";
require_once "File.php";
require_once "GPBMetadata/Messages.php";

class RPCRequest {
    private int $id;

    public function __construct(
        private string $method,
        private array $params
    ) {
        $this->id = random_int(0, 10000); // TODO: UUIDv4
    }

    public function serialize(): string {
        return json_encode([
            'id' => $this->id,
            'method' => $this->method,
            'params' => $this->params,
        ]);
    }
}

class RPCResponse {
    public function __construct(
        public int $id,
        public mixed $result,
        public ?string $error = null,
    ) {
    }

    public static function deserialize(string $response) {
        $decoded = json_decode($response, true);

        return new RPCResponse(
            $decoded['id'],
            $decoded['result'],
            $decoded['error'] ?? null
        );
    }
}

class RPC {
    private $socket;

    function __construct(string $address) {
        [$host, $port] = explode(':', $address, 2);
        $errno = 0;
        $error = '';
        $socket = fsockopen($address, (int) $port, $errno, $error);

        if ($socket === false) {
            throw new \RuntimeException(
                sprintf(
                    'Could not open socket %s:%s: %d: %s',
                    $address,
                    $port,
                    $errno,
                    $error
                )
            );
        }

        $this->socket = $socket;
    }

    public function send(RPCRequest $request): RPCResponse {
        fwrite($this->socket, $request->serialize());
        $result = fgets($this->socket);

        if ($result === false) {
            $this->close();
            throw new \RuntimeException('Could not write to socket');
        }

        return RPCResponse::deserialize($result);
    }

    public function close() {
        if (fclose($this->socket) === false) {
            throw new \RuntimeException('Could not close socket');
        }
    }
}

class Dispatcher {
    private $in;
    private $out;
    private $err;

    public function __construct() {
        $this->in = fopen('php://stdin', 'r');
        $this->out = fopen('php://stdout', 'w');
        $this->err = fopen('php://stderr', 'w');
    }

    public function __desctruct() {
        fclose($this->in);
        fclose($this->out);
        fclose($this->err);
    }

    public function handle() {
        try {
            foreach ($this->messages() as $msg) {
                $req = $this->deserializeRequest($msg);

                // Формируем ответ. Тут будет логика приложения.
                $resp = $this->formResponse(
                    200,
                    ['Content-Type' => 'application/json'],
                    "{\"method\": \"{$req->getMethod()}\"}"
                );

                // Отвечаем Go-процессу.
                $this->send($resp->serializeToString());
            }
        } catch (\Throwable $e) {
            $this->error($e->getMessage(), $e->getTraceAsString());
        }
    }

    private function messages(): iterable {
        while (($line = fgets($this->in)) !== false) {
            $line = rtrim($line, PHP_EOL);

            if ($line === "exit") {
                break;
            }

            $len = (int) $line;
            $msg = '';

            while (($data = fread($this->in, min($len, 2048))) !== false) {
                $msg .= $data;
                $len -= strlen($data);

                if ($len <= 0) {
                    yield $msg;
                    break;
                }
            }
        }
    }

    private function deserializeRequest(string $msg): \Request {
        $req = new \Request();
        $req->mergeFromString($msg);

        return $req;
    }

    private function formResponse(int $statusCode, array $headers, string $body): \Response {
        return (new \Response())
            ->setStatusCode($statusCode)
            ->setHeaders($headers)
            ->setBody($body)
        ;
    }

    private function send(string $data): void {
        fwrite($this->out, strlen($data)."\n");

        // FIXME: 2048 здесь указывать неверно, т.к. это количество символов, а
        // должно быть байт.
        foreach (str_split($data, 2048) as $spl) {
            fwrite($this->out, $spl);
        }
    }

    private function error(string $error, string $context = null): void {
        fwrite($this->err, "\033[1;31mError from PHP: $error\033[0m\n");

        if ($context !== null) {
            $context = preg_replace('/\n/', "\n         ", $context);
            fwrite($this->err, "\033[1mContext:\033[0m {$context}\n");
        }
    }
}

(new Dispatcher())->handle();

?>
