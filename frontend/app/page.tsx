export default function Home() {
  return (
    <main className="mx-auto flex min-h-[calc(100vh-73px)] max-w-6xl items-center px-6 py-16">
      <section className="max-w-3xl">
        <p className="mb-4 text-sm font-semibold uppercase tracking-[0.2em] text-[var(--accent)]">
          Технический фундамент
        </p>
        <h1 className="text-4xl font-bold tracking-tight sm:text-6xl">
          Тренировки и прогресс — в одном понятном месте.
        </h1>
        <p className="mt-6 max-w-2xl text-lg leading-8 text-[var(--muted)]">
          GymTracker AI находится на первом этапе разработки. Сейчас готовы базовые web- и API-приложения;
          программы, тренировки и AI Coach будут добавляться отдельными проверенными этапами.
        </p>
        <div className="mt-10 inline-flex rounded-full border border-[var(--border)] bg-[var(--surface)] px-4 py-2 text-sm text-[var(--muted)]">
          Foundation stage · без авторизации и предметных данных
        </div>
      </section>
    </main>
  );
}
