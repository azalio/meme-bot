export default {
  async fetch(request, env) {
    // Разрешаем только POST запросы
    if (request.method !== 'POST') {
      return new Response('Use POST with JSON { "prompt": "your text" }', {
        headers: { 'Content-Type': 'text/plain' },
        status: 405
      });
    }

    try {
      // Получаем данные из запроса
      const { prompt, steps } = await request.json();

      // Проверяем обязательное поле prompt
      if (!prompt || typeof prompt !== 'string' || prompt.length < 1 || prompt.length > 2048) {
        return new Response('Invalid "prompt": must be a string between 1 and 2048 characters', {
          headers: { 'Content-Type': 'text/plain' },
          status: 400
        });
      }

      // Проверяем steps (если есть)
      if (steps && (typeof steps !== 'number' || steps < 1 || steps > 8)) {
        return new Response('Invalid "steps": must be an integer between 1 and 8', {
          headers: { 'Content-Type': 'text/plain' },
          status: 400
        });
      }

      // Генерируем изображение через модель
      const response = await env.AI.run(
        '@cf/black-forest-labs/flux-1-schnell',
        {
          prompt,
          steps: steps || 4 // Используем значение по умолчанию
        }
      );

      // Конвертируем base64 в бинарные данные
      const binaryString = atob(response.image);
      const img = Uint8Array.from(binaryString, (m) => m.codePointAt(0));

      // Возвращаем изображение с правильными заголовками
      return new Response(img, {
        headers: {
          'Content-Type': 'image/jpeg',
          'Cache-Control': 'public, max-age=3600'
        }
      });

    } catch (err) {
      // Обработка ошибок
      return new Response(`Error: ${err.message}`, {
        headers: { 'Content-Type': 'text/plain' },
        status: 500
      });
    }
  }
};
