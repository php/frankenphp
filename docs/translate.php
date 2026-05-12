<?php

# update all translations to match the english docs
# usage: php docs/translate.php [specific-file.md]
# needs: php with openssl and Gemini api key

const MODEL = 'gemini-2.5-flash';
const SLEEP_SECONDS_BETWEEN_REQUESTS = 10;
const LANGUAGES = [
    'cn' => 'Chinese',
    'fr' => 'French',
    'ja' => 'Japanese',
    'pt-br' => 'Portuguese (Brazilian)',
    'ru' => 'Russian',
    'tr' => 'Turkish',
];
const SYSTEM_PROMPT = <<<'MD'
You are translating the FrankenPHP server documentation from English to other languages.

You will receive the English version (authoritative) and a translation (possibly incomplete or incorrect). Your task is to produce a corrected and complete translation in the target language.

### Rules

* **Structure:** You must not change the structure of the document (headings, code blocks, etc.).
* **Code:** You must not translate code, only comments inside code blocks.
* **General Links:** You must not translate link URLs, only the link text.
* **php.net Exception:** You must rewrite all `php.net` URLs to use their language-agnostic format by removing the language segment (e.g., change `https://www.php.net/manual/en/function.echo.php` to `https://www.php.net/manual/function.echo.php`).
* **Anchors:** You may translate anchors to translation pages (e.g., `config.md#translated-anchor`), but keep existing anchors as they are.
* **Content Integrity:** You must not add or remove any content; only translate what is present.
* **Accuracy:** You must ensure that the translation is accurate and faithful to the original meaning.
* **Style:** You must write in a natural and fluent style appropriate for technical documentation.
* **Terminology:** You must use the correct terminology for technical terms in the target language; do not translate technical terms if you are unsure.
* **Output:** You must not include any explanations or notes, only the translated document.
MD;

function makeGeminiRequest(string $systemPrompt, string $userPrompt, string $model, string $apiKey, int $reties = 2): string
{
    $url = "https://generativelanguage.googleapis.com/v1beta/models/$model:generateContent";
    $body = json_encode([
        "systemInstruction" => ["parts" => [["text" => $systemPrompt]]],
        "contents" => [
            ["role" => "user", "parts" => [["text" => $userPrompt]]],
        ],
    ]);

    $response = @file_get_contents($url, false, stream_context_create([
        'http' => [
            'method' => 'POST',
            'header' => "Content-Type: application/json\r\nX-Goog-Api-Key: $apiKey\r\nContent-Length: " . strlen($body) . "\r\n",
            'content' => $body,
            'timeout' => 300,
        ]
    ]));
    $generatedDocs = json_decode($response, true)['candidates'][0]['content']['parts'][0]['text'] ?? '';

    if (!$response || !$generatedDocs) {
        print_r(error_get_last());
        print_r($response);

        if ($reties > 0) {
            echo "Retrying... ($reties retries left)\n";
            sleep(SLEEP_SECONDS_BETWEEN_REQUESTS);

            return makeGeminiRequest($systemPrompt, $userPrompt, $model, $apiKey, $reties - 1);
        }

        exit(1);
    }

    return $generatedDocs;
}

function createPrompt(string $language, string $englishFile, string $currentTranslation): array
{
    $languageName = LANGUAGES[$language];
    $userPrompt = <<<MD
Here is the english version of the document:

```markdown
$englishFile
```

Here is the current translation in $languageName:

```markdown
$currentTranslation
```

Here is the corrected and completed translation in $languageName:

```markdown
MD;

    return [SYSTEM_PROMPT, $userPrompt];
}

function sanitizeMarkdown(string $markdown): string
{
    if (str_starts_with($markdown, '```markdown')) {
        $markdown = substr($markdown, strlen('```markdown'));
    }

    $markdown = rtrim($markdown, '`');

    return trim($markdown) . "\n";
}

$fileToTranslate = $argv;
array_shift($fileToTranslate);
$fileToTranslate = array_map(fn($filename) => trim($filename), $fileToTranslate);
$apiKey = $_SERVER['GEMINI_API_KEY'] ?? $_ENV['GEMINI_API_KEY'] ?? '';
if (!$apiKey) {
    echo 'Enter gemini api key ($GEMINI_API_KEY): ';
    $apiKey = trim(fgets(STDIN));
}

$files = array_filter(scandir(__DIR__), fn($filename) => str_ends_with($filename, '.md'));
foreach ($files as $file) {
    $englishFile = file_get_contents(__DIR__ . "/$file");
    if ($fileToTranslate && !in_array($file, $fileToTranslate)) {
        continue;
    }

    foreach (LANGUAGES as $language => $languageName) {
        echo "Translating $file to $languageName\n";
        $currentTranslation = file_get_contents(__DIR__ . "/$language/$file") ?: '';
        [$systemPrompt, $userPrompt] = createPrompt($language, $englishFile, $currentTranslation);
        $markdown = makeGeminiRequest($systemPrompt, $userPrompt, MODEL, $apiKey);

        echo "Writing translated file to $language/$file\n";
        file_put_contents(__DIR__ . "/$language/$file", sanitizeMarkdown($markdown));

        echo "sleeping to avoid rate limiting...\n";
        sleep(SLEEP_SECONDS_BETWEEN_REQUESTS);
    }
}
