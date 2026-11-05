/** @type {import('tailwindcss').Config} */
// Палитра плоской тёмной темы (макеты A3). Шкалы slate и indigo переопределены целиком:
// компоненты продолжают писать text-slate-400 / bg-indigo-500, а цвета меняются здесь.
export default {
    content: [
        "./index.html",
        "./src/**/*.{svelte,js,ts,jsx,tsx}",
    ],
    theme: {
        extend: {
            colors: {
                // Нейтральные серые с холодным оттенком: фон #0E1014, панели #12151A/#161920
                slate: {
                    50: '#F4F5F8',
                    100: '#E8EAF0',
                    200: '#D2D6DE',
                    300: '#B9BECA',
                    400: '#A3A9B6',
                    500: '#7D8492',
                    600: '#5B6272',
                    700: '#3A3F4B',
                    800: '#222631',
                    900: '#161920',
                    950: '#0E1014',
                },
                // Акцент: приглушённый фиолетовый (кнопка «Играть», выделение)
                indigo: {
                    50: '#EEECFF',
                    100: '#E4E0FF',
                    200: '#C9C1FF',
                    300: '#B7ADFF',
                    400: '#8C7CF0',
                    500: '#6D5BD0',
                    600: '#5A49B8',
                    700: '#463895',
                    800: '#2F2668',
                    900: '#2A2450',
                    950: '#1A1638',
                },
            },
            fontFamily: {
                // Inter встроен в сборку (@fontsource-variable/inter); иероглифы — системные шрифты
                sans: ['"Inter Variable"', 'Inter', '"Segoe UI"', 'system-ui', '-apple-system', '"Noto Sans"', '"Microsoft YaHei"', '"Yu Gothic UI"', 'sans-serif'],
            },
        },
    },
    plugins: [],
}
