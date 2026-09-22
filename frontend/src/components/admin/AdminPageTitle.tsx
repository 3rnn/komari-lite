import type { ReactNode } from "react";

export default function AdminPageTitle({
  children,
  description,
}: {
  children: ReactNode;
  description?: ReactNode;
}) {
  return (
    <div className="min-w-0">
      <h1 className="text-xl font-semibold leading-7 text-foreground">
        {children}
      </h1>
      {description ? (
        <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      ) : null}
    </div>
  );
}

export function AdminSectionTitle({ children }: { children: ReactNode }) {
  return (
    <h2 className="text-base font-semibold leading-6 text-foreground">
      {children}
    </h2>
  );
}
