<?php

namespace Runner;

/**
 * Прослойка между приложением PHP и corerunner. Открывает stdin, слушает
 * proto-сообщения от coreruner, обрабатывает их и отдает ответ в stdout.
 */
final class Dispatcher
{
    /** @var resource */
    private mixed $in;

    /** @var resource */
    private mixed $out;

    /** @var resource */
    private mixed $err;

    public function __construct()
    {
        $this->in = fopen('php://stdin', 'r');
        $this->out = fopen('php://stdout', 'w');
        $this->err = fopen('php://stderr', 'w');
    }

    public function __destruct()
    {
        fclose($this->in);
        fclose($this->out);
        fclose($this->err);
    }

    /**
     * Запуск блокирующего слушателя сообщений от corerunner. $handler получает
     * бинарное сообщение текстом, которое нужно десериализовать.
     */
    public function run(\Closure $handler): void
    {
        // Сообщаем серверу, что готовы принимать запросы.
        fwrite($this->out, "ok\n");

        try {
            foreach ($this->messages() as $msg) {
                $this->send($handler($msg));
            }
        } catch (\Throwable $e) {
            $this->error($e->getMessage(), $e->getTraceAsString());
        }
    }

    private function messages(): iterable
    {
        while (($line = fgets($this->in)) !== false) {
            $line = rtrim($line, "\n");

            if (strlen($line) === 0) {
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

    private function send(string $data): void
    {
        fwrite($this->out, strlen($data)."\n");

        foreach (str_split($data, 2048) as $spl) {
            fwrite($this->out, $spl);
        }
    }

    private function error(string $error, string $context = null): void
    {
        fwrite($this->err, "\033[1;31mError from PHP: $error\033[0m\n");

        if ($context !== null) {
            $context = preg_replace('/\n/', "\n         ", $context);
            fwrite($this->err, "\033[1mContext:\033[0m {$context}\n");
        }
    }
}
