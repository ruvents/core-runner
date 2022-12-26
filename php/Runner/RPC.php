<?php

namespace Runner;

final class RPC {
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
