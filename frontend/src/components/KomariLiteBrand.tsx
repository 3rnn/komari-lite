type KomariLiteBrandProps = {
  size?: "sm" | "md";
  className?: string;
};

const sizeClasses = {
  sm: {
    brand: "text-xl",
    lite: "text-[13px]",
  },
  md: {
    brand: "text-2xl",
    lite: "text-base",
  },
};

export default function KomariLiteBrand({
  size = "md",
  className = "",
}: KomariLiteBrandProps) {
  const classes = sizeClasses[size];

  return (
    <span
      className={`inline-flex items-baseline whitespace-nowrap leading-none ${className}`}
      aria-label="Komari Lite"
    >
      <span className={`${classes.brand} font-bold leading-none`}>Komari</span>
      <span
        className={`${classes.lite} ml-1.5 font-semibold leading-none text-[var(--green-9)]`}
      >
        Lite
      </span>
    </span>
  );
}
