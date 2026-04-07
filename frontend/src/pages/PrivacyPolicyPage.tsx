import { Link } from 'react-router-dom'
import { SnakeLogo } from '@/components/ui/Icons'

export function PrivacyPolicyPage() {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-surface-950 text-gray-900 dark:text-slate-100">
      {/* Header */}
      <header className="border-b border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900">
        <div className="mx-auto flex h-14 max-w-3xl items-center justify-between px-4">
          <Link to="/" className="flex items-center gap-2">
            <SnakeLogo size={28} />
            <span className="text-sm font-bold tracking-wide">MelloWPN</span>
          </Link>
          <Link
            to="/"
            className="text-sm text-gray-500 dark:text-slate-400 hover:text-primary-500 transition-colors"
          >
            ← На главную
          </Link>
        </div>
      </header>

      {/* Content */}
      <main className="mx-auto max-w-3xl px-4 py-10">
        <h1 className="mb-8 text-2xl font-bold">Политика конфиденциальности</h1>

        <Section title="1. Общие положения">
          <p>1.1. Настоящая Политика конфиденциальности (далее — «Политика») регулирует порядок обработки и защиты информации, которую Пользователь передаёт при использовании сервиса (далее — «Сервис»).</p>
          <p>1.2. Используя Сервис, Пользователь подтверждает своё согласие с условиями Политики. Если Пользователь не согласен с условиями — он обязан прекратить использование Сервиса.</p>
        </Section>

        <Section title="2. Сбор информации">
          <p>2.1. Сервис может собирать следующие типы данных:</p>
          <ul>
            <li>идентификаторы аккаунта (логин, ID, никнейм и т.п.);</li>
            <li>техническую информацию (IP-адрес, данные о браузере, устройстве и операционной системе);</li>
            <li>историю взаимодействий с Сервисом.</li>
          </ul>
          <p>2.2. Сервис не требует от Пользователя предоставления паспортных данных, документов, фотографий или другой личной информации, кроме минимально необходимой для работы.</p>
        </Section>

        <Section title="3. Использование информации">
          <p>3.1. Сервис может использовать полученную информацию исключительно для:</p>
          <ul>
            <li>обеспечения работы функционала;</li>
            <li>связи с Пользователем (в том числе для уведомлений и поддержки);</li>
            <li>анализа и улучшения работы Сервиса.</li>
          </ul>
        </Section>

        <Section title="4. Передача информации третьим лицам">
          <p>4.1. Администрация не передаёт полученные данные третьим лицам, за исключением случаев:</p>
          <ul>
            <li>если это требуется по закону;</li>
            <li>если это необходимо для исполнения обязательств перед Пользователем (например, при работе с платёжными системами);</li>
            <li>если Пользователь сам дал на это согласие.</li>
          </ul>
        </Section>

        <Section title="5. Хранение и защита данных">
          <p>5.1. Данные хранятся в течение срока, необходимого для достижения целей обработки.</p>
          <p>5.2. Администрация принимает разумные меры для защиты данных, но не гарантирует абсолютную безопасность информации при передаче через интернет.</p>
        </Section>

        <Section title="6. Отказ от ответственности">
          <p>6.1. Пользователь понимает и соглашается, что передача информации через интернет всегда сопряжена с рисками.</p>
          <p>6.2. Администрация не несёт ответственности за утрату, кражу или раскрытие данных, если это произошло по вине третьих лиц или самого Пользователя.</p>
        </Section>

        <Section title="7. Изменения в Политике">
          <p>7.1. Администрация вправе изменять условия Политики без предварительного уведомления.</p>
          <p>7.2. Продолжение использования Сервиса после внесения изменений означает согласие Пользователя с новой редакцией Политики.</p>
        </Section>

        <div className="mt-8 rounded-xl border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 px-5 py-4 text-sm text-gray-500 dark:text-slate-400">
          В решении вопросов поможет:{' '}
          <a
            href="https://t.me/Mellow_support"
            target="_blank"
            rel="noopener noreferrer"
            className="font-semibold text-primary-500 hover:underline"
          >
            @Mellow_support
          </a>
        </div>
      </main>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-8">
      <h2 className="mb-3 text-base font-bold text-gray-900 dark:text-slate-100">{title}</h2>
      <div className="space-y-2 text-sm leading-relaxed text-gray-600 dark:text-slate-400 [&_ul]:mt-2 [&_ul]:space-y-1 [&_ul]:pl-5 [&_ul]:list-disc [&_ul]:marker:text-primary-500">
        {children}
      </div>
    </section>
  )
}
