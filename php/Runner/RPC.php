<?php

namespace Runner;

/**
 * RPC-клиент для отправки команд Go процессу.
 */
final class RPC
{
    private $socket;
    private string $address;

    public function __construct(string $address)
    {
        $this->socket = null;
        $this->address = $address;
    }

    public function send(RPCRequest $request): RPCResponse
    {
        // Ленивое подключение.
        $socket = $this->connect();

        fwrite($socket, $request->serialize());
        $result = fgets($socket);

        if ($result === false) {
            $this->close();
            throw new \RuntimeException('Could not write to socket');
        }

        return RPCResponse::deserialize($result);
    }

    public function close(): void
    {
        if ($this->socket === null) {
            return;
        }

        if (fclose($this->socket) === false) {
            throw new \RuntimeException('Could not close socket');
        }
    }

    private function connect(): mixed
    {
        if ($this->socket !== null) {
            return $this->socket;
        }

        [$host, $port] = explode(':', $this->address, 2);
        $errno = 0;
        $error = '';
        $socket = fsockopen($host, (int) $port, $errno, $error);

        if ($socket === false) {
            throw new \RuntimeException(
                sprintf(
                    'Could not open socket %s:%s: %d: %s',
                    $this->address,
                    $port,
                    $errno,
                    $error
                )
            );
        }

        return $this->socket = $socket;
    }
}
