<?php

# update all translations to match the english docs
# usage: php docs/translate.php [specific-file.md]
# needs: php with openssl and gemini api key

const MODEL = 'gemini-2.5-flash';
const SLEEP_SECONDS_BETWEEN_REQUESTS = 5;
const LANGUAGES = [
    'cn' => 'Chinese',
    'fr' => 'French',
    'ja' => 'Japanese',
    'pt-br' => 'Portuguese (Brazilian)',
    'ru' => 'Russian',
    'tr' => 'Turkish',
];

function makeGeminiRequest(string $systemPrompt, string $userPrompt, string $model, string $apiKey): string
{
    $url = "https://generativelanguage.googleapis.com/v1beta/models/$model:generateContent";
    $body = json_encode([
        "contents" => [
            ["role" => "model", "parts" => ['text' => $systemPrompt]],
            ["role" => "user", "parts" => ['text' => $userPrompt]]
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
        exit(1);
    }

    return $generatedDocs;
}

function createPrompt(string $language, string $englishFile, string $currentTranslation): array
{
    $systemPrompt = <<<PROMPT
        You are translating the docs of the FrankenPHP server from english to other languages.
        You will receive the english version (authoritative) and a translation (possibly incomplete or incorrect).
        Your task is to produce a corrected and complete translation in the target language.
        You must strictly follow these rules:
        - You must not change the structure of the document (headings, code blocks, etc.)
        - You must not translate code, only comments and strings inside the code.
        - You must not translate links to other documentation pages, only the link text.
        - You must not add or remove any content, only translate what is present.
        - You must ensure that the translation is accurate and faithful to the original meaning.
        - You must write in a natural and fluent style, appropriate for technical documentation.
        - You must use the correct terminology for technical terms in the target language, don't translate if unsure.
        - You must not include any explanations or notes, only the translated document.
        PROMPT;

    $languageName = LANGUAGES[$language];
    $userPrompt = <<<PROMPT
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
        PROMPT;

    return [$systemPrompt, $userPrompt];
}

function sanitizeMarkdown(string $markdown): string
{
    if (str_starts_with($markdown, '```markdown')) {
        $markdown = substr($markdown, strlen('```markdown'));
    }
    $markdown = rtrim($markdown, '`');
    return trim($markdown) . "\n";
}

$fileToTranslate = $argv[1] ?? '';
$apiKey = $_SERVER['GEMINI_API_KEY'] ?? $_ENV['GEMINI_API_KEY'] ?? '';
if (!$apiKey) {
    echo 'Enter gemini api key ($GEMINI_API_KEY): ';
    $apiKey = trim(fgets(STDIN));
}

$files = array_filter(scandir(__DIR__), fn($filename) => str_ends_with($filename, '.md'));
foreach ($files as $file) {
    $englishFile = file_get_contents(__DIR__ . "/$file");
    if ($fileToTranslate && $fileToTranslate !== $file && "$fileToTranslate.md" !== $file) {
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
